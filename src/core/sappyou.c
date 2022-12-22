#include <malloc.h>
#include <string.h>
#include <time.h>

#include "../../include/core.h"

int sappyou_init(Sappyou *sappyou) {
    uint64_t timestamp = time(NULL);
    sappyou->created_ts = timestamp;
    sappyou->modified_ts = timestamp;
    sappyou->size = 0;
    sappyou->removed_cnt = 0;
    sappyou->contents = NULL;
    sappyou->file = NULL;
    return 0;
}

int sappyou_free(Sappyou *sappyou) {
    for (uint64_t i = 0; i < sappyou->size; i++) {
        free(sappyou->contents[i].name);
        free(sappyou->contents[i].alias);
        free(sappyou->contents[i].description);
    }
    free(sappyou->contents);
    if (sappyou->file != NULL) {
        fclose(sappyou->file);
    }
    return 0;
}

int sappyou_load(Sappyou *sappyou) {
    if (sappyou->file == NULL) {
        fprintf(stderr, "Failed to load sappyou: file not specified\n");
        return 1;
    }
    uint16_t signature[4];
    rewind(sappyou->file);
    fread(signature, 2, 4, sappyou->file);
    if (memcmp(signature, SAPPYOU_SIG, 8) != 0) {
        fprintf(stderr, "Failed to load sappyou: invalid signature\n");
        return 1;
    }
    fread(&sappyou->created_ts, 8, 1, sappyou->file);
    fread(&sappyou->modified_ts, 8, 1, sappyou->file);
    fread(&sappyou->size, 8, 1, sappyou->file);
    sappyou->removed_cnt = 0;
    sappyou->contents = malloc(sappyou->size * sizeof(Tanzaku));
    size_t max_string_len = SIZE_MAX;
    for (uint64_t i = 0; i < sappyou->size; i++) {
        if (fgetc(sappyou->file) != 0) {
            sappyou->contents[i].id = i + 1;
            fread(&sappyou->contents[i].created_ts, 8, 1, sappyou->file);
            fread(&sappyou->contents[i].modified_ts, 8, 1, sappyou->file);
            getdelim(&sappyou->contents[i].name, &max_string_len, 0, sappyou->file);
            getdelim(&sappyou->contents[i].alias, &max_string_len, 0, sappyou->file);
            getdelim(&sappyou->contents[i].description, &max_string_len, 0, sappyou->file);
        } else {
            sappyou->contents[i].id = 0;
            sappyou->removed_cnt++;
        }
    }
    return 0;
}

int sappyou_save(Sappyou *sappyou) {
    if (sappyou->file == NULL) {
        fprintf(stderr, "Failed to save sappyou: file not specified\n");
        return 1;
    }
    rewind(sappyou->file);
    fwrite(SAPPYOU_SIG, 2, 4, sappyou->file);
    fwrite(&sappyou->created_ts, 8, 1, sappyou->file);
    fwrite(&sappyou->modified_ts, 8, 1, sappyou->file);
    fwrite(&sappyou->size, 8, 1, sappyou->file);
    fflush(sappyou->file);
    for (uint64_t i = 0; i < sappyou->size; i++) {
        if (sappyou->contents[i].id != 0) {
            fputc(-1, sappyou->file);
            fwrite(&sappyou->contents[i].created_ts, 8, 1, sappyou->file);
            fwrite(&sappyou->contents[i].modified_ts, 8, 1, sappyou->file);
            fputs(sappyou->contents[i].name, sappyou->file);
            fputc(0, sappyou->file);
            fputs(sappyou->contents[i].alias, sappyou->file);
            fputc(0, sappyou->file);
            fputs(sappyou->contents[i].description, sappyou->file);
            fputc(0, sappyou->file);
        } else {
            fputc(0, sappyou->file);
        }
    }
    fflush(sappyou->file);
    return 0;
}

int sappyou_open(Sappyou *sappyou, const char *path) {
    sappyou->file = fopen(path, "r+b");
    if (sappyou->file == NULL) {
        fprintf(stderr, "Failed to dump sappyou: failed to open file '%s'\n", path);
        return 1;
    }
    return sappyou_load(sappyou);
}

int sappyou_dump(Sappyou *sappyou, const char *path) {
    sappyou->file = fopen(path, "w+b");
    if (sappyou->file == NULL) {
        fprintf(stderr, "Failed to dump sappyou: failed to open file '%s'\n", path);
        return 1;
    }
    return sappyou_save(sappyou);
}

int tanzaku_add(Sappyou *sappyou, const char *name, const char *alias, const char *description) {
    if (sappyou->size == -1) {
        fprintf(stderr, "Failed to add tanzaku: sappyou is full\n");
        return 1;
    }
    Tanzaku newbie;
    newbie.created_ts = time(NULL);
    newbie.modified_ts = newbie.created_ts;
    size_t name_size = strlen(name),
            alias_size = strlen(alias),
            description_size = strlen(description);
    newbie.name = malloc(name_size + 1);
    strcpy(newbie.name, name);
    newbie.name[name_size] = 0;
    newbie.alias = malloc(alias_size + 1);
    strcpy(newbie.alias, alias);
    newbie.alias[alias_size] = 0;
    newbie.description = malloc(description_size + 1);
    strcpy(newbie.description, description);
    newbie.description[description_size] = 0;
    sappyou->size++;
    newbie.id = sappyou->size;
    sappyou->contents = realloc(sappyou->contents, sappyou->size * sizeof(Tanzaku));
    sappyou->contents[sappyou->size - 1] = newbie;
    sappyou->modified_ts = newbie.created_ts;
    return 0;
}

int tanzaku_rem_by_id(Sappyou *sappyou, uint64_t tanzaku_id) {
    if (tanzaku_id == 0) {
        fprintf(stderr, "Failed to remove tanzaku: got zero ID\n");
        return 1;
    }
    for (uint64_t i = 0; i < sappyou->size; i++) {
        if (sappyou->contents[i].id == tanzaku_id) {
            sappyou->modified_ts = time(NULL);
            sappyou->contents[i].id = 0;
            sappyou->removed_cnt++;
            return 0;
        }
    }
    fprintf(stderr, "Failed to remove tanzaku: target tanzaku does not exist\n");
    return 1;
}

int tanzaku_rem_by_name(Sappyou *sappyou, const char *name) {
    for (uint64_t i = 0; i < sappyou->size; i++) {
        if (strcmp(sappyou->contents[i].name, name) == 0) {
            if (sappyou->contents[i].id != 0) {
                sappyou->modified_ts = time(NULL);
                sappyou->contents[i].id = 0;
                sappyou->removed_cnt++;
                return 0;
            } else {
                fprintf(stderr, "Failed to remove tanzaku: target tanzaku is already removed\n");
                return 1;
            }
        }
    }
    fprintf(stderr, "Failed to remove tanzaku: target tanzaku does not exist\n");
    return 1;
}

int tanzaku_rem_by_alias(Sappyou *sappyou, const char *alias) {
    for (uint64_t i = 0; i < sappyou->size; i++) {
        if (strcmp(sappyou->contents[i].alias, alias) == 0) {
            if (sappyou->contents[i].id != 0) {
                sappyou->modified_ts = time(NULL);
                sappyou->contents[i].id = 0;
                sappyou->removed_cnt++;
                return 0;
            } else {
                fprintf(stderr, "Failed to remove tanzaku: target tanzaku is already removed\n");
                return 1;
            }
        }
    }
    fprintf(stderr, "Failed to remove tanzaku: target tanzaku does not exist\n");
    return 1;
}
